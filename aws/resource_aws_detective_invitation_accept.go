package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAwsDetectiveInvitationAccept() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceDetectiveInvitationAcceptCreate,
		ReadWithoutTimeout:   resourceDetectiveInvitationAcceptRead,
		UpdateWithoutTimeout: resourceDetectiveInvitationAcceptUpdate,
		DeleteWithoutTimeout: resourceDetectiveInvitationAcceptDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"graph_arn": {
				Type:     schema.TypeString,
				Required: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceDetectiveInvitationAcceptCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.AcceptInvitationInput{
		GraphArn: aws.String(d.Get("graph_arn").(string)),
	}

	var err error
	err = resource.RetryContext(ctx, 4*time.Minute, func() *resource.RetryError {
		_, err = conn.AcceptInvitationWithContext(ctx, input)
		if err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if isResourceTimeoutError(err) {
		_, err = conn.AcceptInvitationWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error accepting invitation: %w", err))
	}

	d.SetId(*input.GraphArn)

	return resourceDetectiveInvitationAcceptRead(ctx, d, meta)
}

func resourceDetectiveInvitationAcceptRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.ListInvitationsInput{}

	resp, err := conn.ListInvitationsWithContext(ctx, input)

	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading Detective member invitation (%s): %w", d.Id(), err))
	}

	var invitationDetails *detective.MemberDetail = nil
	for _, invitation := range resp.Invitations {
		if *invitation.GraphArn == d.Id() {
			invitationDetails = invitation
			break
		}
	}

	if invitationDetails == nil {
		d.SetId("")
		return diag.FromErr(fmt.Errorf("No invitation was found for graph (%s): %w", d.Id()))
	}

	d.Set("status", invitationDetails.Status)
	d.Set("graph_arn", invitationDetails.GraphArn)
	return nil
}

func resourceDetectiveInvitationAcceptDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.DisassociateMembershipInput{
		GraphArn: aws.String(d.Id()),
	}

	_, err := conn.DisassociateMembershipWithContext(ctx, input)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting graph membership for (%s): %w", d.Id(), err))
	}

	return nil
}

func resourceDetectiveInvitationAcceptUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	diagnostics = resourceDetectiveInvitationAcceptDelete(ctx, d, meta)
	if diagnostics != nil {
		return diagnostics
	}

	diagnostics = resourceDetectiveInvitationAcceptCreate(ctx, d, meta)
	if diagnostics != nil {
		return diagnostics
	}

	return resourceDetectiveInvitationAcceptRead(ctx, d, meta)
}
