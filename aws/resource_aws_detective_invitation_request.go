package aws

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const IdSeparator = "/"

func resourceAwsDetectiveInvitationRequest() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceDetectiveInvitationRequestCreate,
		ReadWithoutTimeout:   resourceDetectiveInvitationRequestRead,
		UpdateWithoutTimeout: resourceDetectiveInvitationRequestUpdate,
		DeleteWithoutTimeout: resourceDetectiveInvitationRequestDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"graph_arn": {
				Type:     schema.TypeString,
				Required: true,
			},
			"account": {
				Type:     schema.TypeString,
				Required: true,
			},
			"email": {
				Type:     schema.TypeString,
				Required: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"message": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"disable_email_notification": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func resourceDetectiveInvitationRequestCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.CreateMembersInput{
		GraphArn:                 aws.String(d.Get("graph_arn").(string)),
		DisableEmailNotification: aws.Bool(d.Get("disable_email_notification").(bool)),
		Accounts: []*detective.Account{
			{
				AccountId:    aws.String(d.Get("account").(string)),
				EmailAddress: aws.String(d.Get("email").(string)),
			},
		},
	}

	if v, ok := d.GetOk("message"); ok {
		input.Message = aws.String(v.(string))
	}

	var err error
	var res *detective.CreateMembersOutput
	err = resource.RetryContext(ctx, 4*time.Minute, func() *resource.RetryError {
		res, err = conn.CreateMembersWithContext(ctx, input)
		if err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if isResourceTimeoutError(err) {
		res, err = conn.CreateMembersWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error inviting member: %w", err))
	}

	id := *input.GraphArn + IdSeparator + *input.Accounts[0].AccountId
	d.SetId(id)

	return resourceDetectiveInvitationRequestRead(ctx, d, meta)
}

func resourceDetectiveInvitationRequestRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	invitationInfo := strings.Split(d.Id(), IdSeparator)

	input := &detective.GetMembersInput{
		GraphArn: aws.String(invitationInfo[0]),
		AccountIds: []*string{
			aws.String(invitationInfo[1]),
		},
	}

	resp, err := conn.GetMembersWithContext(ctx, input)

	if tfawserr.ErrCodeEquals(err, detective.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Graph or member does not seem to exist for invitation (%s), removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		d.SetId("")
		return diag.FromErr(fmt.Errorf("error reading Detective member invitation (%s): %w", d.Id(), err))
	}

	if len(resp.MemberDetails) == 0 {
		d.SetId("")
		return nil // diag.FromErr(fmt.Errorf("error reading Detective member invitation (%s)", d.Id()))
	}

	d.Set("graph_arn", resp.MemberDetails[0].GraphArn)
	d.Set("account", resp.MemberDetails[0].AccountId)
	d.Set("email", resp.MemberDetails[0].EmailAddress)
	d.Set("status", resp.MemberDetails[0].Status)
	return nil
}

func resourceDetectiveInvitationRequestDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	invitationInfo := strings.Split(d.Id(), IdSeparator)

	input := &detective.DeleteMembersInput{
		GraphArn: aws.String(invitationInfo[0]),
		AccountIds: []*string{
			aws.String(invitationInfo[1]),
		},
	}

	err := resource.RetryContext(ctx, 4*time.Minute, func() *resource.RetryError {
		_, err := conn.DeleteMembersWithContext(ctx, input)

		if err != nil {
			return resource.NonRetryableError(err)
		}
		return nil
	})

	if isResourceTimeoutError(err) {
		_, err = conn.DeleteMembersWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error delete Detective graph (%s): %w", d.Id(), err))
	}

	return nil
}

func resourceDetectiveInvitationRequestUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	diagnostics = resourceDetectiveInvitationRequestDelete(ctx, d, meta)
	if diagnostics != nil {
		return diagnostics
	}

	diagnostics = resourceDetectiveInvitationRequestCreate(ctx, d, meta)
	if diagnostics != nil {
		return diagnostics
	}

	return resourceMacie2AccountRead(ctx, d, meta)
}
